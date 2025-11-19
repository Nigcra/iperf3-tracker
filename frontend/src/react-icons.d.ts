declare module 'react-icons/fi' {
  import { FC, SVGProps } from 'react';
  
  export interface IconBaseProps extends SVGProps<SVGSVGElement> {
    size?: string | number;
    color?: string;
    title?: string;
  }
  
  export const FiHome: FC<IconBaseProps>;
  export const FiActivity: FC<IconBaseProps>;
  export const FiServer: FC<IconBaseProps>;
  export const FiMenu: FC<IconBaseProps>;
  export const FiX: FC<IconBaseProps>;
  export const FiTrendingUp: FC<IconBaseProps>;
  export const FiTrendingDown: FC<IconBaseProps>;
  export const FiAlertTriangle: FC<IconBaseProps>;
  export const FiUsers: FC<IconBaseProps>;
  export const FiTrash2: FC<IconBaseProps>;
  export const FiDatabase: FC<IconBaseProps>;
  export const FiAlertCircle: FC<IconBaseProps>;
}
